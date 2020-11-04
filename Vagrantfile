# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure('2') do |config|
  config.vm.box = 'ubuntu/bionic64'
  config.vm.box_check_update = false
  config.vm.host_name = 'opentelemetry-collector-contrib'
  config.vm.network :private_network, ip: "192.168.33.33"

  config.vm.provider 'virtualbox' do |vb|
    vb.gui = false
    vb.memory = 4096
    vb.name = 'opentelemetry-collector-contrib'
  end
  config.vm.provision 'file', source: 'vagrant', destination: 'vagrant'
  config.vm.provision 'shell', path: 'vagrant/provision.sh'
end
